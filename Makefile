.PHONY: all build clean test run

all: build

build:
	go build -o marionette.exe ./src/cmd/marionette/

run: build
	./marionette.exe

clean:
	rm -f marionette.exe
	rm -f .marionette.db

test: build
	./marionette.exe --db .test.db init
	./marionette.exe --db .test.db index testdata
	./marionette.exe --db .test.db query --format markdown
	./marionette.exe --db .test.db stats
	rm -f .test.db .test.db-journal

lint:
	./marionette.exe lint .

help:
	./marionette.exe --help
