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
      
      const data = await this.githubGraphqlService.queryTrendingRepos();
      
      if (!data?.data?.search?.edges?.length) {
        throw new Error('No repository data received');
      }

      const repo = data.data.search.edges[0].node;
      
      await this.GithubTrendSchema.create({
        name: repo.name,
        owner: repo.owner.login,
        description: repo.description,
        starCount: repo.stargazerCount,
        forkCount: repo.forkCount,
        language: repo.primaryLanguage?.name,
        openIssuesCount: repo.issues.totalCount,
        latestRelease: repo.releases.edges[0]?.node || null,
        url: repo.url,
        homepageUrl: repo.homepageUrl,
        readme: repo.readme?.text,
        fetchedAt: new Date(),
      });

      this.logger.log(`Successfully saved trending data for ${repo.name}`);
    } catch (error) {
      this.logger.error(`Failed to fetch and save trending data: ${error.message}`);
    }
  }
} 