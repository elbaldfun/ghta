import { Injectable } from '@nestjs/common';
import { InjectModel } from '@nestjs/mongoose';
import { Model } from 'mongoose';
import { GithubTrendDto } from './dto/github-trend.dto';
import { GithubTrend } from './schemas/github-trend.schema';

@Injectable()
export class GithubTrendService {
  constructor(
    @InjectModel(GithubTrend.name) private GithubTrendSchema: Model<GithubTrend>,
  ) {}

  async fetchTrendingRepos(filters?: {
    minStars?: number;
    maxStars?: number;
    language?: string;
    minIssues?: number;
    maxIssues?: number;
    limit?: number;
    sort?: { [key: string]: 1 | -1 };
  }): Promise<GithubTrend[]> {
    const query = this.GithubTrendSchema.find();

    // Apply filters if provided
    if (filters) {
      if (filters.minStars || filters.maxStars) {
        const starFilter: any = {};
        if (filters.minStars) starFilter.$gte = filters.minStars;
        if (filters.maxStars) starFilter.$lte = filters.maxStars;
        query.where('starCount').equals(starFilter);
      }

      if (filters.language) {
        query.where('language').equals(filters.language);
      }

      if (filters.minIssues || filters.maxIssues) {
        const issuesFilter: any = {};
        if (filters.minIssues) issuesFilter.$gte = filters.minIssues;
        if (filters.maxIssues) issuesFilter.$lte = filters.maxIssues;
        query.where('openIssuesCount').equals(issuesFilter);
      }

      // Apply sorting
      if (filters.sort) {
        query.sort(filters.sort);
      } else {
        query.sort({ starCount: -1 }); // Default sort by stars descending
      }

      // Apply limit
      if (filters.limit) {
        query.limit(filters.limit);
      } else {
        query.limit(100); // Default limit
      }
    }

    const trendingRepos = await query.exec();
    return trendingRepos;
  }
}
