# mongodb

1. 创建数据库
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
