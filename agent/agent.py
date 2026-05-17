import os
from dotenv import load_dotenv
from livekit.agents import Agent, AgentSession, JobContext, WorkerOptions, cli
from livekit.agents.voice.room_io import RoomOptions
from livekit.plugins import openai
from livekit.plugins.openai.realtime.realtime_model import TurnDetection

load_dotenv()

INSTRUCTIONS = (
    "You are Sol, a helpful smart home AI assistant. "
    "Help the user control their home, answer questions, and manage their devices."
)


class SolAgent(Agent):
    def __init__(self):
        super().__init__(instructions=INSTRUCTIONS)

    async def on_enter(self):
        self.session.generate_reply(
            instructions="Greet the user briefly as Sol, their smart home assistant."
        )


async def entrypoint(ctx: JobContext):
    await ctx.connect()

    session = AgentSession(
        llm=openai.realtime.RealtimeModel.with_azure(
            azure_deployment=os.environ["AZURE_DEPLOYMENT"],
            azure_endpoint=os.environ["AZURE_OPENAI_ENDPOINT"],
            api_key=os.environ["AZURE_OPENAI_KEY"],
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
        agent=SolAgent(),
        room_options=RoomOptions(),
    )


if __name__ == "__main__":
    cli.run_app(WorkerOptions(entrypoint_fnc=entrypoint))
