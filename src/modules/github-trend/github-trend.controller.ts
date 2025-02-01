import { Controller, Get, Query } from '@nestjs/common';
import { GithubTrendService } from './github-trend.service';
import { GithubTrend } from './schemas/github-trend.schema';
import { ApiOperation, ApiQuery, ApiResponse } from '@nestjs/swagger';

@Controller('trending')
export class GithubTrendController {
  constructor(private readonly githubTrendService: GithubTrendService) {}

  @Get()
  @ApiOperation({
    summary: '获取 GitHub 仓库趋势',
    description: '获取 GitHub 趋势仓库列表，支持多种过滤和排序选项'
  })
  @ApiQuery({
    name: 'stars',
    required: false,
    type: String,
    description: 'star 数量，格式为 starts:1000..2000，starts:>1000，starts:<1000，starts:1000'
  })
  @ApiQuery({
    name: 'language',
    required: false,
    type: String,
    description: '编程语言，如 Python, JavaScript, TypeScript, Java, C++, C# 等'
  })
  @ApiQuery({
    name: 'issues',
    required: false,
    type: String,
    description: 'issue 数量，格式为 issues:100..200，issues:>100，issues:<100，issues:100'
  })
  @ApiQuery({
    name: 'limit',
    required: false,
    type: Number,
    description: '返回结果数量限制，默认 10'
  })
  @ApiQuery({
    name: 'sort',
    required: false,
    type: String,
    description: '排序方式，格式为 field:order，例如 stars:desc, stars:asc'
  })
  @ApiResponse({
    status: 200,
    description: '成功获取趋势仓库列表',
    type: GithubTrend,
    isArray: true
  })
  async getTrendingRepos(
    @Query('stars') stars?: string,
    @Query('issues') issues?: number,
    @Query('language') language?: string,
    @Query('limit') limit?: number,
    @Query('sort') sort?: string,
  ): Promise<{data: GithubTrend[]}> {
    const filters: any = {};
    
    if (stars) filters.stars = stars;
    if (language) filters.language = language;
    if (issues) filters.issues = issues;
    if (limit) filters.limit = Number(limit);
    
    if (sort) {
      const [field, order] = sort.split(':');
      filters.sort = { [field]: order === 'desc' ? -1 : 1 };
    }

    const data = await this.githubTrendService.fetchTrendingRepos(filters);
    return {data};
  }
}
