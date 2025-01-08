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

  @Cron(CronExpression.EVERY_10_MINUTES)
  // @Cron(CronExpression.)
  async fetchTrendingRepos() {
    try {
      this.logger.log('Starting to fetch trending repositories...');
      // const start = 10000;
      // 1. 100000..400000
      // 2. 50000..100000
      // 3. 30000..50000
      // 4. 10000..30000
      // 5. 8000..10000
      // 6. 7000..8000
      // 7. 6000..7000
      // 8. 5000..6000
      // 9. 4000..5000
      // 10. 3000..4000
      // 11. 2000..3000
      const start = 200000;
      const end = 800000;
      // for (let i = end; i >= start; i -= 20000) {
      //   const range = `${i - 20000}..${i}`;
      //   const datas = await this.githubGraphqlService.fetchAllTrendingRepos(range);
      //   this.logger.log(`try to fetch trending repos, range: ${range}`);
      // }
      // const range = '7000..9000';
      // const datas = await this.githubGraphqlService.fetchAllTrendingRepos(range);
      // this.logger.log(`Finished to fetch trending repos, range: ${range}`);

    } catch (error) {
      this.logger.error(error);
      this.logger.error(`Failed to fetch and save trending data: ${error.message}`);
    }
  }
} 