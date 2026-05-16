import os
from dotenv import load_dotenv
from livekit.agents import AutoSubscribe, JobContext, WorkerOptions, cli
from livekit.agents.multimodal import MultimodalAgent
from livekit.plugins import openai

load_dotenv()

INSTRUCTIONS = (
    "You are Sol, a helpful smart home AI assistant. "
    "Help the user control their home, answer questions, and manage their devices."
)


async def entrypoint(ctx: JobContext):
    await ctx.connect(auto_subscribe=AutoSubscribe.AUDIO_ONLY)

    model = openai.realtime.RealtimeModel.with_azure(
        azure_deployment=os.environ["AZURE_DEPLOYMENT"],
        azure_endpoint=os.environ["AZURE_OPENAI_ENDPOINT"],
        api_key=os.environ["AZURE_OPENAI_KEY"],
        api_version=os.environ.get("AZURE_API_VERSION", "2025-04-01-preview"),
        voice="alloy",
        instructions=INSTRUCTIONS,
        turn_detection=openai.realtime.ServerVadOptions(
            threshold=0.5,
            prefix_padding_ms=300,
            silence_duration_ms=500,
        ),
    )

    agent = MultimodalAgent(model=model)
    agent.start(ctx.room)


if __name__ == "__main__":
    cli.run_app(WorkerOptions(entrypoint_fnc=entrypoint))
