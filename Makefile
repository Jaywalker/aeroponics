all:
	GOOS=linux GOARCH=arm go build

clean:
	rm aeroponics
	
install: all
	scp aeroponics 10.0.0.21:/home/jaywalker/
