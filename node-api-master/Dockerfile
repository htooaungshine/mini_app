FROM node:18
ARG NODE_ENV
ENV TZ=Asia/Yangon

# Install necessary packages, including dirmngr, gnupg, and curl
RUN apt-get update && apt-get install -y gnupg dirmngr curl tzdata

# Set timezone
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone && \
    dpkg-reconfigure --frontend noninteractive tzdata

# Update package list after adding keys
RUN apt-get update

###
#4. Application
###

EXPOSE 1339
RUN mkdir -p /usr/src/app
WORKDIR /usr/src/app
COPY package.json /usr/src/app/package.json

RUN npm install

COPY . /usr/src/app


RUN echo "Environment is ${NODE_ENV}"
ENV NODE_ENV ${NODE_ENV}

ENTRYPOINT ["node", "server.js"]