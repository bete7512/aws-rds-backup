./build-run.sh

./check.sh

./stop.sh


tail -f logs/rds-backup.log



sudo cp systemd-service /etc/systemd/system/rds-backup.service
sudo systemctl daemon-reload
sudo systemctl enable rds-backup.service
sudo systemctl start rds-backup.service


sudo systemctl status rds-backup.service  
sudo systemctl stop rds-backup.service   
sudo journalctl -u rds-backup.service  
sudo systemctl restart rds-backup.service