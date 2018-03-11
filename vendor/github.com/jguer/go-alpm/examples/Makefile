all: alpm-installed alpm-search alpm-updates

alpm-installed: installed.go
	go build -x -o $@ $<

alpm-search: search.go
	go build -x -o $@ $<

alpm-updates: updates.go
	go build -x -o $@ $<

clean:
	rm -f alpm-installed alpm-search alpm-updates
