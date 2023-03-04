FROM golang:1.19

COPY tunnel /app

WORKDIR /app 

RUN apt update && apt install -y iproute2 iperf3 netcat && rm -rf /var/lib/apt/lists/*

RUN go install
RUN go build -o /app/main main.go 

CMD /app/main
