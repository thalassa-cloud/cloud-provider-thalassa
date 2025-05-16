FROM gcr.io/distroless/static:nonroot
EXPOSE 8080 2112

COPY bin/cloud-provider-thalassa_linux_amd64*/cloud-provider-thalassa /cloud-provider-thalassa
ENTRYPOINT ["/cloud-provider-thalassa"]
