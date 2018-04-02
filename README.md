# SETUP

## PREREQISITES

- AWSCLI installed and setup in target machine (CCM).
- This code deployed in go for AWS Lambda.
- Environment variables setup as appropriate.

### AWS INSTALLATION IN CCM/TARGET MACHINE

- yum install epel-release –y
- yum install python-pip –y
- pip install awscli

### CONFIGURE AWS

- aws configure

### DEPLOYMENT BUILD FOR LAMBDA

- GOOS=linux go build -o sync
- zip deployment.zip sync

### UPLOAD ZIP AND SET ENV VARIABLES

- Upload deployment.zip
- Set handler as sync
- Set CCM_IP = {target_machine_ip}
- Set PEM_FILE = {target_machine_pem_file}
- Set USERNAME = {target_machine_username}

- Upload PEM_FILE to the S3 bucket of your choice.

- Set S3 bucket of your choice to be the source for the Lambda fujnction. Event type = ObjectCreated

Thanks. For any queries contact - sandyydk@cisco.com
