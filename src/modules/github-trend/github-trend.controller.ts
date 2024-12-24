import { Controller, Get } from '@nestjs/common';
import { GithubTrendService } from './github-trend.service';
import { GithubTrendDto } from './dto/github-trend.dto';

@Controller('trending')
export class GithubTrendController {
  constructor(private readonly githubTrendService: GithubTrendService) {}

  @Get()
  async getTrendingRepos(): Promise<GithubTrendDto[]> {
    return this.githubTrendService.fetchTrendingRepos();
  }
}
