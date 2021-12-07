APP=password-rotation
SHA=$(git rev-parse --short=8 HEAD)
docker build -t $APP:$SHA -f test-script.Dockerfile .
aws ecr get-login-password --region=us-east-1 |  docker login --username AWS --password-stdin $(aws sts get-caller-identity --output text --query Account).dkr.ecr.us-east-1.amazonaws.com
REMOTE=$(aws sts get-caller-identity --output text --query Account).dkr.ecr.us-east-1.amazonaws.com/$APP:$SHA
docker tag $APP:$SHA $REMOTE
docker push $REMOTE