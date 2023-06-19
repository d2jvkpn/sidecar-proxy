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


build:
	echo ">>> git branch: $(git_branch)"
	mkdir -p target
	go build -o target/main -ldflags="-X main.build_time=$(build_time) \
	  -X main.git_branch=$(git_branch) -X main.git_commit_id=unknown" main.go

docker-build:
	bash deployments/build.sh dev

docker-build-local:
	BuildLocal=true bash deployments/build.sh dev

run:
	go run main.go serve --config=configs/sidecar-proxy.yaml --addr=:9000

create-user_md5:
	go run main.go create-user --method=md5

create-user_bcrypt:
	go run main.go create-user --method=bcrypt
