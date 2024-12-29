import { Prop, Schema, SchemaFactory } from '@nestjs/mongoose';
import { Document } from 'mongoose';


@Schema()
export class Release {
    @Prop({ required: true })
    name: string;

    @Prop({ required: true })
    tagName: string;

    @Prop({ required: true })
    isPrerelease: boolean;

    @Prop({ required: true })
    isLatest: boolean;

    @Prop({ required: true })
    isDraft: boolean;

    @Prop({ required: true })
    publishedAt: Date;
}

@Schema()
export class repoTopics {
    @Prop({ required: true })
    name: string;

    @Prop({ required: true })
    url: string;
}

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

  @Prop({ type: [Release], required: false})
  top5Release: Release[];

  @Prop({ type: [repoTopics], required: false })
  repoTopics: repoTopics[];

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