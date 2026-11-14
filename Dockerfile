FROM gcr.io/distroless/static-debian12:nonroot

COPY BeMovin /bin/BeMovin

USER nonroot

CMD ["/bin/BeMovin"]
