1. 将一个字段的一条记录置空

db.users.updateOne(
    { _id: 1 }, // 查询条件，这里根据 _id 找到要更新的文档
    { $set: { hobbies: [] } } // 使用 $set 操作符将 hobbies 字段设置为空数组
);
1. 将一个字段的所有记录置空
db.users.updateMany(
    {}, // 空查询条件表示匹配集合中的所有文档
    { $set: { hobbies: [] } } // 使用 $set 操作符将 hobbies 字段设置为空数组
);