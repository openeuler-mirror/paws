FROM python:3.8-slim

ADD algorithm /sir_vpa/algorithm
ADD config /sir_vpa/config
ADD core /sir_vpa/core
ADD utils /sir_vpa/utils
ADD *.py /sir_vpa/
ADD *.txt /sir_vpa/
ADD *.yaml /sir_vpa/
RUN cd /sir_vpa && pip install -r requirements.txt

ENV PYTHONUNBUFFERED=1
ENV PYTHONPATH "${PYTHONPATH}:/sir_vpa/"

CMD [ "python", "/sir_vpa/main.py"]
