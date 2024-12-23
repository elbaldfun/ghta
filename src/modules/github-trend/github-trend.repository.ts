import { Injectable } from '@nestjs/common';
import { InjectModel } from '@nestjs/mongoose';
import { Model } from 'mongoose';
import { GitHubTrendDto } from './dto/github-trend.dto';

@Injectable()
export class GitHubTrendRepository {
  constructor(@InjectModel('GitHubTrend') private readonly model: Model<GitHubTrendDto>) {}

  async saveTrendingRepo(repo: GitHubTrendDto): Promise<GitHubTrendDto> {
    const createdRepo = new this.model(repo);
    return createdRepo.save();
  }

  async findAll(): Promise<GitHubTrendDto[]> {
    return this.model.find().exec();
  }
}
