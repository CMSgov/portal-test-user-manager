
Note:  the APP variable must match the name of the Dockerfile ($APP.Dockerfile) and the ECR repo that hosts the application images.
These instructions tag the image with both the Git hash and 'latest', which is used by the scheduled runner to pull the correct image from ECR when running the task. 

With creds for `aws-cms-oit-iusg-spe-cmcs-macbis-test` (156322662943): 

```
APP=password-rotation
SHA=$(git rev-parse --short=8 HEAD)

aws ecr get-login-password --region=us-east-1 |  docker login --username AWS --password-stdin $(aws sts get-caller-identity --output text --query Account).dkr.ecr.us-east-1.amazonaws.com
REMOTE=$(aws sts get-caller-identity --output text --query Account).dkr.ecr.us-east-1.amazonaws.com/$APP
docker build -t $REMOTE:$SHA -t $REMOTE:latest -f $APP.Dockerfile .
for t in latest $SHA; do docker push "$REMOTE:${t}"; done
```