# Use alpine as base and copy the already-built binary
FROM alpine:3.19

# Create a non-root user
RUN adduser -D -u 1000 appuser

# Copy the pre-built binary from build context
COPY bin/planner /planner

# Make sure it's executable and owned by appuser
RUN chmod +x /planner && chown appuser:appuser /planner

# Switch to non-root user
USER appuser

ENTRYPOINT ["/planner"]
