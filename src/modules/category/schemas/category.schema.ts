import { Prop, Schema, SchemaFactory } from '@nestjs/mongoose';
import { Document, Types } from 'mongoose';


@Schema({ timestamps: true, versionKey: false, id: true })
export class Category extends Document {
  @Prop({ required: true })
  name: string;

  @Prop({ required: false })
  description: string;

  @Prop({ required: true, type: Types.ObjectId })
  parentId: Types.ObjectId;

  @Prop({ required: true })
  level: number;

  @Prop({ required: true })
  path: string;
}

export const CategorySchema = SchemaFactory.createForClass(Category); 