# make

# include envfile
# export $(shell sed 's/=.*//' envfile)

current = $(shell pwd)

build_time = $(shell date +'%FT%T%:z')
git_branch = $(shell git rev-parse --abbrev-ref HEAD)
# git_commit_id = $(shell git rev-parse --verify HEAD)
# git_commit_time = $(shell git log -1 --format="%at" | xargs -I{} date -d @{} +%FT%T%:z)

# git_tree_state="clean"
# uncommitted=$(git status --short)
# unpushed=$(git diff origin/$git_branch..HEAD --name-status)
# -- [[ ! -z "$uncommitted$unpushed" ]] && git_tree_state="dirty"

run:
	mkdir -p target
	go build -o target/main -ldflags="-w -s -X main.build_time=$(build_time) \
	  -X main.git_branch=$(git_branch) -X main.git_commit_id=unknown" main.go
	./target/main

build:
	echo ">>> git branch: $(git_branch)"
	mkdir -p target
	go build -o target/main -ldflags="-X main.build_time=$(build_time) \
	  -X main.git_branch=$(git_branch) -X main.git_commit_id=unknown" main.go

docker-build:
	BuildLocal=true bash deployments/build.sh dev
