FROM debian:stable-slim

COPY BeMovin /bin/BeMovin

CMD ["/bin/BeMovin"]
