import logging
import os

import aiohttp
from dotenv import load_dotenv
from livekit.agents import Agent, AgentSession, JobContext, WorkerOptions, cli, function_tool
from livekit.agents.voice.room_io import RoomOptions
from livekit.plugins import openai
from livekit.plugins.openai.realtime.realtime_model import TurnDetection

load_dotenv()

logger = logging.getLogger(__name__)

SOL_API_URL = os.environ.get("SOL_API_URL", "http://sol-core:8080")

INSTRUCTIONS = (
    "You are Sol, a helpful smart home AI assistant. "
    "Help the user control their home, answer questions, and manage their devices. "
    "When the user asks to control a device: first call discover_devices to find it, "
    "then check_device_online, then control_device. "
    "Always confirm what you did after controlling a device."
)


def _device_id_from_room(room_name: str) -> str:
    # room name format: voice-{device_uuid}-{8_char_suffix}
    # e.g. voice-2094a9e6-8287-4ba7-b0ff-48c6d070d778-0a34217d
    return room_name[6:-9]  # strip "voice-" prefix and "-{8chars}" suffix


async def _call_tool(device_id: str, tool: str, arguments: str) -> str:
    async with aiohttp.ClientSession() as session:
        async with session.post(
            f"{SOL_API_URL}/api/internal/voice/tools",
            json={"device_id": device_id, "tool": tool, "arguments": arguments},
        ) as resp:
            return await resp.text()


class SolAgent(Agent):
    def __init__(self, device_id: str):
        super().__init__(instructions=INSTRUCTIONS)
        self._device_id = device_id

    async def on_enter(self):
        self.session.generate_reply(
            instructions="Greet the user briefly as Sol, their smart home assistant."
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
        agent=SolAgent(device_id=device_id),
        room_options=RoomOptions(),
    )
    logger.info("agent session started")


if __name__ == "__main__":
    cli.run_app(WorkerOptions(entrypoint_fnc=entrypoint))
