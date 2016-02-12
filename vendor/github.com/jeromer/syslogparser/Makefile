SUBPACKAGES=. rfc3164 rfc5424
help:
	@echo "Available targets:"
	@echo "- tests: run tests"
	@echo "- installdependencies: installs dependencies declared in dependencies.txt"
	@echo "- clean: cleans directory"
	@echo "- benchmarks: run benchmarks"

installdependencies:
	@cat dependencies.txt | xargs go get

tests: installdependencies
	@for pkg in $(SUBPACKAGES); do cd $$pkg && go test -i && go test ; cd -;done

clean:
	find . -type 'f' -name '*.test' -print | xargs rm -f

benchmarks:
	@for pkg in $(SUBPACKAGES); do cd $$pkg && go test -gocheck.b ; cd -;done
