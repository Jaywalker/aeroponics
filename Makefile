all:
	GOOS=linux GOARCH=arm go build

clean:
	rm aeroponics
