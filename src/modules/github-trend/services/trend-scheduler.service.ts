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

  @Cron(CronExpression.EVERY_10_SECONDS)
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
      const end = 400000;
      for (let i = end; i >= start; i -= 10000) {
        const range = `${i - 10000}..${i}`;
        const datas = await this.githubGraphqlService.fetchAllTrendingRepos(range);
        this.logger.log(`try to fetch trending repos, range: ${range}, datas length: ${datas.length}`);
        await this.parseTrendingReposIntoDB(datas);
        await new Promise(resolve => setTimeout(resolve, 300));
      }

    } catch (error) {
      this.logger.error(error);
      this.logger.error(`Failed to fetch and save trending data: ${error.message}`);
    }
  }

  async parseTrendingReposIntoDB(datas: any[]) {
    this.logger.debug(`parseTrendingReposIntoDB, start to parse, datas length: ${datas.length}`);
    for (const data of datas) {
      if (data?.data?.search?.edges?.length < 1) {
        this.logger.error(`if result is ${!data?.data?.search?.edges?.length}`);
          throw new Error('No repository data received');
        }
  
      for (const edge of data.data.search.edges) {
        // const repo = data.data.search.edges[0].node;
        const repo = edge.node;
        const repoNameID = repo.url.split('https://github.com/')[1]; 
        const existingRepo = await this.GithubTrendSchema.findOne({ repoNameID: repoNameID });
        
        if (existingRepo) {
          await this.GithubTrendSchema.updateOne({repoNameID: repoNameID}, {
            owner: repo.owner.login,
            name: repo.name,
            repoNameID: repoNameID,
            description: repo.description,
            starCount: repo.stargazerCount,
            forkCount: repo.forkCount,
            forkFromRepo: repo.forkFromRepository?.name,
            language: repo.primaryLanguage?.name,
            openIssuesCount: repo.issues.totalCount,
            latestRelease: repo.releases.edges[0]?.node || null,
            url: repo.url,
            homepageUrl: repo.homepageUrl,
            readme: repo.readme?.text,
            fetchedAt: new Date(),
          })
          this.logger.log(`Successfully updated trending data for ${repo.name}`);
        } else {
          await this.GithubTrendSchema.create({
            owner: repo.owner.login,
            name: repo.name,
            repoNameID: repoNameID,
            description: repo.description,
            starCount: repo.stargazerCount,
            forkCount: repo.forkCount,
            forkFromRepo: repo.forkFromRepository?.name,
            language: repo.primaryLanguage?.name,
            openIssuesCount: repo.issues.totalCount,
            latestRelease: repo.releases.edges[0]?.node || null,
            url: repo.url,
            homepageUrl: repo.homepageUrl,
            readme: repo.readme?.text,
            fetchedAt: new Date(),
          });
          this.logger.log(`Successfully saved trending data for ${repo.name}`);
        }
      }
    }
  }
} 