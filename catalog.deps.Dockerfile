# The base image which contain rm and sed command
FROM cirros AS builder

# Copy catalog files
COPY catalog /configs

# Remove odf bundle from the files
RUN rm -f /configs/odf.yaml

# Remove odf bundle details from the index file
RUN sed -i '1,11d' /configs/index.yaml


# The base image is expected to contain
# /bin/opm (with a serve subcommand) and /bin/grpc_health_probe
FROM quay.io/operator-framework/opm:v1.26.0

# Configure the entrypoint and command
ENTRYPOINT ["/bin/opm"]
CMD ["serve", "/configs"]

# Copy declarative config root into image at /configs
COPY --from=builder /configs /configs

# Set DC-specific label for the location of the DC root directory in the image
LABEL operators.operatorframework.io.index.configs.v1=/configs
