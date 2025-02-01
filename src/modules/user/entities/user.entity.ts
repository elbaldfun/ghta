import { Prop, Schema, SchemaFactory } from '@nestjs/mongoose';
import { Document } from 'mongoose';
import { LoginType } from '../dto/user.dto';

@Schema({
  timestamps: true, // 自动管理 createdAt 和 updatedAt
  collection: 'users', // 指定集合名称
})
export class User extends Document {
  @Prop({
    required: true,
    unique: true,
  })
  id: string;

  @Prop({
    required: true,
  })
  name: string;

  @Prop({
    required: true,
    unique: true,
  })
  email: string;

  @Prop({
    required: true,
    enum: LoginType,
    type: String,
  })
  loginType: LoginType;

  @Prop()
  createdAt: Date;

  @Prop()
  updatedAt: Date;
}

export const UserSchema = SchemaFactory.createForClass(User);

// 创建索引
UserSchema.index({ email: 1 }, { unique: true });
UserSchema.index({ id: 1 }, { unique: true });
