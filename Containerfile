FROM scratch
copy collect_server /
ENTRYPOINT ["/collect_server"]
