# README

This repo contains an application (the "password-rotation" app) and a terraform module ("password-rotation"). The app runs as a scheduled task on ECS Fargate. The MACFin team instantiates the module in their own terraform and deploys it in a MACFin AWS account. The input for the app is an xlsx spreadsheet, stored in the S3 bucket created by the module.

The app rotates passwords for accounts in the IDM portals. For each user, the app logs in, changes the user password, if necessary, and logs out. Whenever testers need a user to test a scenario, they are assured the user will always have a valid password. Testers can focus on their tests and never have to worry about rotating passwords. Testers must not manually change the password in the portal.

The password-rotation module includes a README.md that explains how to configure the module.

For user information and a troubleshooting guide, please see the [password-rotation application confluence page](https://confluenceent.cms.gov/x/SGbzDg).
