import { Prop, Schema, SchemaFactory } from '@nestjs/mongoose';
import { Document } from 'mongoose';

@Schema({ timestamps: true })
export class GithubTrend extends Document {
  @Prop({ required: true })
  name: string;

  @Prop({ required: true })
  repoNameID: string;

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
  top5Release: {
    name: string;
    tagName: string;
    isPrerelease: boolean;
    isLatest: boolean;
    isDraft: boolean;
    publishedAt: Date;
  }[];

  @Prop({ type: Object })
  repoTopics: {
    name: string;
    url: string;
  }[];

  @Prop({ required: true })
  url: string;

  @Prop()
  homepageUrl: string;

  @Prop()
  forkFromRepo: string;

  @Prop()
  readme: string;

  @Prop({ required: true })
  fetchedAt: Date;
}

export const GithubTrendSchema = SchemaFactory.createForClass(GithubTrend); 