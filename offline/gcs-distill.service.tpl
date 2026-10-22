[Unit]
Description=GCS Distill 2 offline control service
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
WorkingDirectory=@@PACKAGE_DIR@@
EnvironmentFile=-/etc/gcs-distill/environment
ExecStart=@@PACKAGE_DIR@@/bin/gcs-distill-server --config /etc/gcs-distill/config.toml
Restart=on-failure
RestartSec=5
TimeoutStopSec=45

[Install]
WantedBy=multi-user.target
