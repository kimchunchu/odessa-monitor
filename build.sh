go build -o $1 ./src/main.go
cp $1 /opt/odessa/$1

sudo systemctl restart $1
sudo systemctl status $1
