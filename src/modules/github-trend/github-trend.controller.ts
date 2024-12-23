import { Controller, Get } from '@nestjs/common';
import { GitHubTrendService } from './github-trend.service';
import { GitHubTrendDto } from './dto/github-trend.dto';

@Controller('trending')
export class GitHubTrendController {
  constructor(private readonly githubTrendService: GitHubTrendService) {}

  @Get()
  async getTrendingRepos(): Promise<GitHubTrendDto[]> {
    return this.githubTrendService.fetchTrendingRepos();
  }
}
