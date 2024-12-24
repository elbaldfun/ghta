import { Module } from '@nestjs/common';
import { GithubTrendController } from './github-trend.controller';
import { GithubTrendService } from './github-trend.service';
import { MongooseModule } from '@nestjs/mongoose';
import { ScheduleModule } from '@nestjs/schedule';
import { GithubTrend, GithubTrendSchema } from './schemas/github-trend.schema';
import { GithubGraphqlService } from './services/github-graphql.service';
import { TrendSchedulerService } from './services/trend-scheduler.service';
import { ConfigModule } from '@nestjs/config';

@Module({
  imports: [
    ConfigModule,
    ScheduleModule.forRoot(),
    MongooseModule.forFeature([
      { name: GithubTrend.name, schema: GithubTrendSchema },
    ]),
  ],
  controllers: [GithubTrendController],
  providers: [GithubTrendService, GithubGraphqlService, TrendSchedulerService],
  exports: [GithubTrendService, GithubGraphqlService, TrendSchedulerService],
})
export class GithubTrendModule {}
