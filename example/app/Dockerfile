FROM python:2.7
RUN mkdir -p /pycode
RUN apt-get update &&\
    apt-get install -y tcpdump fping iperf netcat net-tools
CMD python app.py
COPY . /pycode
WORKDIR /pycode
RUN pip install -r requirements.txt
