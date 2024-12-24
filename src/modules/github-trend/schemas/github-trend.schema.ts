import { Prop, Schema, SchemaFactory } from '@nestjs/mongoose';
import { Document } from 'mongoose';

@Schema({ timestamps: true })
export class GithubTrend extends Document {
  @Prop({ required: true })
  name: string;

  @Prop({ required: true })
  owner: string;

  @Prop()
  description: string;

  @Prop({ required: true })
  starCount: number;

  @Prop()
  forkCount: number;

  @Prop()
  language: string;

  @Prop()
  openIssuesCount: number;

  @Prop({ type: Object })
  latestRelease: {
    name: string;
    tagName: string;
  };

  @Prop({ required: true })
  url: string;

  @Prop()
  homepageUrl: string;

  @Prop()
  readme: string;

  @Prop({ required: true })
  fetchedAt: Date;
}

export const GithubTrendSchema = SchemaFactory.createForClass(GithubTrend); 