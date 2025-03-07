import { Injectable, Logger } from '@nestjs/common';
import { Cron, CronExpression } from '@nestjs/schedule';
import { GithubGraphqlService } from './github-graphql.service';
import { InjectModel } from '@nestjs/mongoose';
import { Model } from 'mongoose';
import { GithubTrend } from '../schemas/github-trend.schema';

@Injectable()
export class TrendSchedulerService {
  private readonly logger = new Logger(TrendSchedulerService.name);

  constructor(
    private readonly githubGraphqlService: GithubGraphqlService,
    @InjectModel(GithubTrend.name) private GithubTrendSchema: Model<GithubTrend>,
  ) {}

  // @Cron(CronExpression.EVERY_5_HOURS)
  @Cron('0 10 08 * * *') // 每天下午14:50运行
  // @Cron(CronExpression.EVERY_30_SECONDS)
  async fetchTrendingRepos() {
    try {
      this.logger.log('Starting to fetch trending repositories...');
      const rangeDict: { start: number; end: number; step: number }[] = [
        { start: 100000, end: 800000, step: 100000 },
        // { start: 300000, end: 400000, step: 100000 },
        // { start: 200000, end: 300000, step: 100000 },
        // { start: 100000, end: 200000, step: 100000 },
        { start: 50000, end: 100000, step: 2000 },
        { start: 30000, end: 50000, step: 100 },
        { start: 10000, end: 30000, step: 50 },
        { start: 9000, end: 10000, step: 20 },
        { start: 8000, end: 9000, step: 20 },
        { start: 7000, end: 8000, step: 10 },
        { start: 6000, end: 7000, step: 5 },
        { start: 5000, end: 6000, step: 5 },
        { start: 4000, end: 5000, step: 5 },
        { start: 3000, end: 4000, step: 5 },
        { start: 2000, end: 3000, step: 5 },
        { start: 1000, end: 2000, step: 5 },
      ]

      for (const range of rangeDict) {
        const { start, end, step } = range;
        for (let i = end; i >= start; i -= step) {
          const range = `${i - step}..${i}`;
          this.logger.log(`Start to fetch trending repos, range: ${range}`);
          const datas = await this.githubGraphqlService.fetchAllTrendingRepos(range);
          this.logger.log(`Finished to fetch trending repos, range: ${range}`);
        }
      }
      // const range = '7000..9000';
      // const datas = await this.githubGraphqlService.fetchAllTrendingRepos(range);
      // this.logger.log(`Finished to fetch trending repos, range: ${range}`);

    } catch (error) {
      this.logger.error(error);
      this.logger.error(`Failed to fetch and save trending data: ${error.message}`);
    }
  }
} 