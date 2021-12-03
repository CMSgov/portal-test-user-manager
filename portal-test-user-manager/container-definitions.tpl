[
  {
    "name": "${app_name}-${environment}-${task_name}",
    "image": "${image}",
    "cpu": 128,
    "memory": 1024,
    "essential": true,
    "portMappings": [],
    "environment": [
      {"name": "S3_BUCKET", "value": "${s3_bucket}"},
      {"name": "S3_KEY", "value": "${s3_key}"},
    ],
    "logConfiguration": {
      "logDriver": "awslogs",
      "secretOptions": null,
      "options": {
        "awslogs-group": "${awslogs_group}",
        "awslogs-region": "${awslogs_region}",
        "awslogs-stream-prefix": "ecs"
      }
    },
    "mountPoints": [],
    "volumesFrom": [],
    "entryPoint": [
            "./scriptRunner.sh"
    ]
  }
]