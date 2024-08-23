set ip=10.0.0.40

docker save kraken:latest > ./kraken.tar

scp kraken.tar matthewweisfeld@%ip%:~/Documents/G3Tech/Server/Email

scp docker-compose.yml matthewweisfeld@%ip%:~/Documents/G3Tech/Server/Email
scp deploy/remoteDeploy.sh matthewweisfeld@%ip%:~/Documents/G3Tech/Server/Email
scp .env matthewweisfeld@%ip%:~/Documents/G3Tech/Server/Email
scp mailu.env matthewweisfeld@%ip%:~/Documents/G3Tech/Server/Email

ssh matthewweisfeld@%ip% chmod +x ~/Documents/G3Tech/Server/Email/remoteDeploy.sh
ssh matthewweisfeld@%ip% ~/Documents/G3Tech/Server/Email/remoteDeploy.sh