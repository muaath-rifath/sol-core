FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY agent.py .

RUN python agent.py download-files

CMD ["python", "agent.py", "start"]
