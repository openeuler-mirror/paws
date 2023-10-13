FROM python:3.8-slim

ADD algorithm /paws/algorithm
ADD config /paws/config
ADD core /paws/core
ADD utils /paws/utils
ADD *.py /paws/
ADD *.txt /paws/
ADD *.yaml /paws/
RUN cd /paws && pip install -r requirements.txt

ENV PYTHONUNBUFFERED=1
ENV PYTHONPATH "${PYTHONPATH}:/paws/"

CMD [ "python", "/paws/main.py"]
