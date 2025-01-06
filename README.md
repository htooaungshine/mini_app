# mini_app

1. Need to build Docker image to run apps.

2. Go to  golang-transaction-master.
   2.1 #docker build -t go:v1 .
   2.2 #cdocker run -d -p 8080:8080 go:v1

3. Go to node-api-master.
   3.1 #docker build -t node:v1 .
   3.2 #docker run -d -p 3000:3000 node:v1

4. And then pull postgresql image.
   4.1 #docker pull postgres:17
