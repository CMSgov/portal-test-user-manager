APP=password-rotation
SHA=$(git rev-parse --short=8 HEAD)

aws ecr get-login-password --region=us-east-1 |  docker login --username AWS --password-stdin $(aws sts get-caller-identity --output text --query Account).dkr.ecr.us-east-1.amazonaws.com
REMOTE=$(aws sts get-caller-identity --output text --query Account).dkr.ecr.us-east-1.amazonaws.com/$APP

** TODO change to $APP.Dockerfile **
docker build -t $REMOTE:$SHA -t $REMOTE:latest -f test-script.Dockerfile .

for t in latest $SHA; do docker push "$REMOTE:${t}"; done
