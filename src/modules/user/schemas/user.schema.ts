import { Prop, Schema, SchemaFactory } from '@nestjs/mongoose';
import { Document } from 'mongoose';

export enum LoginType {
    GOOGLE = 'google',
    GITHUB = 'github',
}

@Schema({ timestamps: true, versionKey: false, id: true })
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
      required: false,
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