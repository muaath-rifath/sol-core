import asyncio
import logging
import os

import aiohttp
from dotenv import load_dotenv
from livekit import api as lk_api
from livekit.agents import Agent, AgentSession, JobContext, WorkerOptions, cli, function_tool
from livekit.agents.voice.room_io import RoomOptions
from livekit.plugins import openai
from livekit.plugins.openai.realtime.realtime_model import TurnDetection

load_dotenv()

logger = logging.getLogger(__name__)

SOL_API_URL = os.environ.get("SOL_API_URL", "http://sol-core:8080")
INTERNAL_SERVICE_TOKEN = os.environ.get("INTERNAL_SERVICE_TOKEN", "")
INACTIVITY_TIMEOUT = 60  # seconds of silence before ending the session

_BASE_INSTRUCTIONS = (
    "You are Joy, a helpful smart home AI assistant. "
    "Help the user control their home, answer questions, and manage their devices. "
    "When the user asks to control a device: first call discover_devices to find it, "
    "then check_device_online, then control_device. "
    "Always confirm what you did after controlling a device."
)


async def _fetch_session_context(device_id: str) -> dict:
    try:
        async with aiohttp.ClientSession() as session:
            async with session.get(
                f"{SOL_API_URL}/api/internal/voice/context",
                params={"device_id": device_id},
                headers={"Authorization": f"Bearer {INTERNAL_SERVICE_TOKEN}"},
            ) as resp:
                if resp.status == 200:
                    return await resp.json()
    except Exception as e:
        logger.warning("failed to fetch session context: %s", e)
    return {}


def _build_instructions(room_name: str, home_name: str) -> str:
    location = ""
    if room_name:
        location = f" You are the Joy device installed in the {room_name}"
        if home_name:
            location += f" at {home_name}"
        location += (
            ". All device discovery and control is already scoped to this room,"
            " so never ask the user which room they mean."
        )
    return _BASE_INSTRUCTIONS + location


def _device_id_from_room(room_name: str) -> str:
    # room name format: voice-{device_uuid}-{8_char_suffix}
    return room_name[6:-9]  # strip "voice-" prefix and "-{8chars}" suffix


async def _call_tool(device_id: str, tool: str, arguments: str) -> str:
    async with aiohttp.ClientSession() as session:
        async with session.post(
            f"{SOL_API_URL}/api/internal/voice/tools",
            json={"device_id": device_id, "tool": tool, "arguments": arguments},
            headers={"Authorization": f"Bearer {INTERNAL_SERVICE_TOKEN}"},
        ) as resp:
            return await resp.text()


class JoyAgent(Agent):
    def __init__(self, device_id: str, instructions: str):
        super().__init__(instructions=instructions)
        self._device_id = device_id

    async def on_enter(self):
        self.session.generate_reply(
            instructions="Greet the user briefly as Joy, their smart home assistant."
        )

    @function_tool
    async def discover_devices(self, query: str) -> str:
        """Find appliances/devices accessible to the user that match a natural language description. Returns a list of matching appliances with their current state."""
        import json
        return await _call_tool(self._device_id, "discover_devices", json.dumps({"query": query}))

    @function_tool
    async def check_device_online(self, appliance_id: str) -> str:
        """Check whether the physical device backing this appliance is reachable. Always call this before control_device."""
        import json
        return await _call_tool(self._device_id, "check_device_online", json.dumps({"appliance_id": appliance_id}))

    @function_tool
    async def get_device_state(self, appliance_id: str) -> str:
        """Return the appliance's current state, e.g. {isOn: true}."""
        import json
        return await _call_tool(self._device_id, "get_device_state", json.dumps({"appliance_id": appliance_id}))

    @function_tool
    async def control_device(self, appliance_id: str, action: str) -> str:
        """Turn an appliance on or off. action must be 'on' or 'off'. Returns ok and message. When ok is false, the command did NOT take effect."""
        import json
        return await _call_tool(self._device_id, "control_device", json.dumps({"appliance_id": appliance_id, "action": action}))


async def entrypoint(ctx: JobContext):
    logger.info("connecting to room %s", ctx.room.name)
    await ctx.connect()
    logger.info("waiting for participant")
    await ctx.wait_for_participant()
    logger.info("participant joined, starting agent session")

    device_id = _device_id_from_room(ctx.room.name)
    logger.info("device_id extracted from room: %s", device_id)

    ctx_data = await _fetch_session_context(device_id)
    room_name = ctx_data.get("room_name", "")
    home_name = ctx_data.get("home_name", "")
    logger.info("session context: room=%s home=%s", room_name, home_name)

    session = AgentSession(
        llm=openai.realtime.RealtimeModel.with_azure(
            azure_deployment=os.environ["AZURE_DEPLOYMENT"],
            azure_endpoint=os.environ["AZURE_OPENAI_ENDPOINT"],
            api_key=os.environ["AZURE_OPENAI_KEY"],
            api_version=os.environ["AZURE_API_VERSION"],
            voice="alloy",
            turn_detection=TurnDetection(
                type="server_vad",
                threshold=0.5,
                prefix_padding_ms=300,
                silence_duration_ms=500,
            ),
        )
    )

    await session.start(
        room=ctx.room,
        agent=JoyAgent(device_id=device_id, instructions=_build_instructions(room_name, home_name)),
        room_options=RoomOptions(),
    )
    logger.info("agent session started")

    # Track last user speech time for inactivity detection.
    last_activity = asyncio.get_event_loop().time()

    def _on_user_state_changed(event):
        nonlocal last_activity
        if event.new_state == "speaking":
            last_activity = asyncio.get_event_loop().time()

    session.on("user_state_changed", _on_user_state_changed)

    # Inactivity watchdog — polls every 10 s.
    while True:
        await asyncio.sleep(10)
        elapsed = asyncio.get_event_loop().time() - last_activity
        if elapsed >= INACTIVITY_TIMEOUT:
            logger.info("inactivity timeout (%.0fs) — ending session", elapsed)
            break

    # Say goodbye, wait for speech to finish, then delete the room so the
    # ESP32 receives a DISCONNECTED event and returns to wake-word mode.
    goodbye_done = asyncio.Event()
    session.once("agent_stopped_speaking", lambda *_: goodbye_done.set())
    session.generate_reply(
        instructions="The user has been inactive. Say a brief goodbye — one short sentence."
    )
    try:
        await asyncio.wait_for(goodbye_done.wait(), timeout=10)
    except asyncio.TimeoutError:
        pass

    lk = lk_api.LiveKitAPI(
        url=os.environ["LIVEKIT_URL"],
        api_key=os.environ["LIVEKIT_API_KEY"],
        api_secret=os.environ["LIVEKIT_API_SECRET"],
    )
    try:
        await lk.room.delete_room(lk_api.DeleteRoomRequest(room=ctx.room.name))
        logger.info("room deleted: %s", ctx.room.name)
    except Exception as e:
        logger.warning("failed to delete room: %s", e)
    finally:
        await lk.aclose()


if __name__ == "__main__":
    cli.run_app(WorkerOptions(entrypoint_fnc=entrypoint, agent_name="joy-voice-agent"))
