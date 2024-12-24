import { Injectable } from '@nestjs/common';
import { InjectModel } from '@nestjs/mongoose';
import { Model } from 'mongoose';
import { GithubTrendDto } from './dto/github-trend.dto';

@Injectable()
export class GithubTrendRepository {
  constructor(@InjectModel('GitHubTrend') private readonly model: Model<GithubTrendDto>) {}

  async saveTrendingRepo(repo: GithubTrendDto): Promise<GithubTrendDto> {
    const createdRepo = new this.model(repo);
    return createdRepo.save();
  }

  async findAll(): Promise<GithubTrendDto[]> {
    return this.model.find().exec();
  }
}
