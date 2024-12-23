import { Injectable } from '@nestjs/common';
import axios from 'axios';
import { GitHubTrendDto } from './dto/github-trend.dto';

@Injectable()
export class GitHubTrendService {
  private readonly GITHUB_TREND_URL = 'https://github.com/trending';

  async fetchTrendingRepos(): Promise<GitHubTrendDto[]> {
    const response = await axios.get(this.GITHUB_TREND_URL);
    // 解析 HTML 并提取数据的逻辑
    // 这里需要使用 Cheerio 或其他库来解析 HTML
    // 省略具体实现，假设我们得到了一个数组
    const trendingRepos: GitHubTrendDto[] = []; // 解析后的数据
    return trendingRepos;
  }
}
