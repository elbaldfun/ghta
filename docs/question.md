# mongodb

1. 安装 mongodb
    ```
     docker run -d --name mongodb --privileged  -p 27017:27017   --restart=always   -v /root/workspaces/middleware/mongodb:/bitnami/mongodb/   -e MONGODB_ROOT_PASSWORD=ghta513!   -e MONGODB_USERNAME=ghta   -e MONGODB_PASSWORD=ghta513!   bitnami/mongodb:8.0.4

    ```


2. 创建数据库
    ```
    use ghta
    ```
2. 设置用户密码并赋予权限
    ```
    db.createUser({
        user: "root",
        pwd: "ghta513!",
        roles: [
            { role: "readWrite", db: "ghta" }
        ]
    });
    ```

# How to login google account 
https://blog.twofei.com/784/

# others stars
https://chrome-stats.com/stats