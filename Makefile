build-task-controller:
	go build -o bin/task-controller ./cmd/taskcontroller

build-counter:
	go build -o bin/counter ./demo/counter/

build-task-controller-image:
	docker build -t task-controller -f Dockerfile .

build-counter-image:
	docker build -t counter -f demo/counter/Dockerfile .
