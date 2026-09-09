[Unit]
Description=gcs-distill control plane
Wants=network-online.target
After=network-online.target docker.service

[Service]
Type=simple
User=root
WorkingDirectory=@@PACKAGE_DIR@@
ExecStart=@@PACKAGE_DIR@@/bin/gcs-distill-server --config @@PACKAGE_DIR@@/config.toml
Restart=on-failure
RestartSec=5
LimitNOFILE=65536
TimeoutStopSec=45

[Install]
WantedBy=multi-user.target
