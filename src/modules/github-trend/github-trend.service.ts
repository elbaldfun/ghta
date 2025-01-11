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
    stars?: string; // Changed to string to handle range format
    language?: string;
    issues?: string; // Changed to string to handle range format
    limit?: number;
    sort?: { [key: string]: 1 | -1 };
  }): Promise<GithubTrend[]> {
    const query = this.GithubTrendSchema.find();

    // Apply filters if provided
    if (filters) {
      // Handle stars range filter
      if (filters.stars) {
        //starts:
        // starts:1000..2000
        if (filters.stars.includes('..')) {
          const [minStars, maxStars] = filters.stars.split('..').map(Number);
          if (!minStars || !maxStars) {
            throw new Error('Invalid stars range');
          }
          query.where('starCount').gte(minStars).lte(maxStars);
        } else if (filters.stars.includes('>')) {
          // starts:>1000
          const minStars = filters.stars.split('>')
          if (minStars.length < 1) {
            throw new Error('Invalid stars range');
          }
          query.where('starCount').gte(Number(minStars[1]));
        } else if (filters.stars.includes('<')) {
          // starts:<1000
          const maxStars = filters.stars.split('<');
          if (maxStars.length < 1) {
            throw new Error('Invalid stars range');
          }
          query.where('starCount').lte(Number(maxStars[1]));
        } else {
          // starts:1000
          query.where('starCount').equals(filters.stars);
        }
      }

      // Handle issues range filter
      if (filters.issues) {
        if (filters.issues.includes('..')) {
          const [minIssues, maxIssues] = filters.issues.split('..').map(Number);
          if (!minIssues || !maxIssues) {
            throw new Error('Invalid issues range');
          }
          query.where('openIssuesCount').gte(minIssues).lte(maxIssues);
        } else if (filters.issues.includes('>')) {
          const minIssues = filters.issues.split('>');
          if (minIssues.length < 1) {
            throw new Error('Invalid issues range');
          }
          query.where('openIssuesCount').gte(Number(minIssues[1]));
        } else if (filters.issues.includes('<')) {
          const maxIssues = filters.issues.split('<');
          if (maxIssues.length < 1) {
            throw new Error('Invalid issues range');
          }
          query.where('openIssuesCount').lte(Number(maxIssues[1]));
        } else {
          query.where('openIssuesCount').equals(filters.issues);
        }
      }

      // Handle language filter
      if (filters.language) {
        query.where('language').equals(filters.language);
      }

      // Apply sorting
      if (filters.sort) {
        query.sort(filters.sort);
      } else {
        query.sort({ starCount: -1 }); // Default sort by stars descending
      }

      // Apply limit
      if (filters.limit) {
        if (filters.limit > 50) {
          throw new Error('Limit must be less than 50');
        }
        query.limit(filters.limit);
      } else {
        query.limit(50); // Default limit
      }
    }

    const trendingRepos = await query.exec();
    return trendingRepos;
  }
}
