import { Module } from '@nestjs/common';
import { GitHubTrendController } from './github-trend.controller';
import { GitHubTrendService } from './github-trend.service';
import { MongooseModule } from '@nestjs/mongoose';
import { GitHubTrendSchema } from './schemas/github-trend.schema';
@Module({
  imports: [
    MongooseModule.forFeature([{ name: 'GitHubTrend', schema: GitHubTrendSchema }]),
  ],
  controllers: [GitHubTrendController],
  providers: [GitHubTrendService],
})
export class GitHubTrendModule {}
