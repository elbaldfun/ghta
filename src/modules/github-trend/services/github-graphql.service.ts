import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import {inspect} from 'util'
import axios from 'axios';
import { InjectModel } from '@nestjs/mongoose';
import { Model } from 'mongoose';
import { time } from 'console';
import { GithubTrend } from '../schemas/github-trend.schema';

@Injectable()
export class GithubGraphqlService {
  private readonly logger = new Logger(GithubGraphqlService.name);
  private readonly apiUrl = 'https://api.github.com/graphql';
  private requestCount = 0;
  private lastResetTime = Date.now();
  private readonly MAX_REQUESTS_PER_HOUR = 5000; // GitHub API 限制

  constructor(
    private configService: ConfigService,
    @InjectModel(GithubTrend.name) private GithubTrendSchema: Model<GithubTrend>,
  ) {}

  async fetchAllTrendingRepos(range: string) {
    // let allTrendingRepos = [];
    let hasNextPage = true;
    let startCursor: string = ""; // 初始化为 ""
    let afterCursor: string = ""; // 初始化为 ""
  
    while (hasNextPage) {
      this.logger.debug(`fetchAllTrendingRepos, start to fetch, range is ${range}, startCursor is ${startCursor}, afterCursor is ${afterCursor}`)
      const response = await this.queryTrendingRepos(range, 30, afterCursor);
      if (response.data.search.pageInfo.startCursor === null) {
        this.logger.debug(`Github response no data for range ${range}, because of response.data.search.pageInfo.startCursor is null`)
        break;
      }
      // 处理返回的数据
      await this.parseTrendingReposIntoDB(response);
      await new Promise(resolve => setTimeout(resolve, 300));
      // const trendingRepos = response.data.search.edges.map(edge => edge.node);
      // allTrendingRepos = [...allTrendingRepos, ...trendingRepos];
  
      // 更新分页信息
      hasNextPage = response.data.search.pageInfo.hasNextPage;
      startCursor = response.data.search.pageInfo.startCursor; // 获取当前游标
      afterCursor = response.data.search.pageInfo.endCursor; // 获取下一个游标
      if (hasNextPage) {
        this.logger.debug(`Current range is ${range}, hasNextPage is ${hasNextPage}, current cursor is ${startCursor}, the next cursor is ${afterCursor}`)
        await new Promise(resolve => setTimeout(resolve, 80));
        startCursor = afterCursor;
      }
    }
  
    // return allTrendingRepos;
  }


  async queryTrendingRepos(range: string, first: number, after: string = ""): Promise<any> {
    try {
      // 检查并重置计数器
      this.checkAndResetRequestCount();
      // 检查是否超出限制
      if (this.requestCount >= this.MAX_REQUESTS_PER_HOUR) {
        throw new Error('Exceeded GitHub API hourly limit');
      }
      let search = "";
      if (after === "") {
        search = `query: "stars:${range}", type: REPOSITORY, first: ${first}`
      } else {
        search = `query: "stars:${range}", type: REPOSITORY, first: ${first}, after: "${after}"`
      }

      this.logger.debug(`Search parameters is ${search}`)
      const query = `
            query {
              search(${search}) {
                pageInfo {
                  hasPreviousPage
                  hasNextPage
                  startCursor
                  endCursor
                } 
                edges {
                  cursor
                  node {
                    ... on Repository {
                      name
                      owner {
                        login
                      }
                      repositoryTopics(first: 20) {
                        edges {
                          node {
                            topic {
                              name
                            }
                            url
                          }
                        }
                      }
                      description
                      stargazerCount
                      forkCount
                      pushedAt
                      primaryLanguage {
                        name
                      }
                      issues(states: OPEN) {
                        totalCount
                      }
                      releases(first: 5, orderBy: {field: CREATED_AT, direction: DESC}) {
                        edges {
                          node {
                            name
                            tagName
                            isPrerelease
                            isLatest
                            isDraft
                            publishedAt
                          }
                        }
                      }
                      url
                      homepageUrl
                      licenseInfo {
                        name
                        nickname
                        description
                        
                      }
                    }
                  }
                }
              }
            }
          `
      this.logger.debug(`Query is ${query}`)
      const response = await axios.post(
        this.apiUrl,
        {

                      //  homepageUrl
                      //  readme: object(expression: "HEAD:README.md") {
                      //   ... on Blob {
                      //     text
                      //   }
                      // }
                      // query: "stars:>300000", type: REPOSITORY, first: 5, after: "Y3Vyc29yOjU="
                      // search(query: "stars:>0", type: REPOSITORY, first: 1) {
                      // query: "stars:>300000", type: REPOSITORY, first: 95, after: "Y3Vyc29yOjU="

                      // query: "stars:300..400", type: REPOSITORY, first: 95, after: "Y3Vyc29yOjU="
          query: query,
        },
        {
          headers: {
            Authorization: `Bearer ${this.configService.get('GITHUB_API_TOKEN')}`,
          },
        }
      );

      this.requestCount++;
      if (response.status != 200) {
        this.logger.error(`Failed to fetch GitHub data: ${response.status}, ${response.data}`)
      }
      
      // this.logger.debug(`Github Graphql API response data: ${inspect(response, {showHidden: true, depth: null})}`)
      this.logger.debug(`Success to fetch GitHub data: ${response.data.data.search?.edges[0]?.node?.url}`);
      return response.data;
    } catch (error) {
      this.logger.error(`Failed to fetch GitHub data: ${error.message}`);
      throw error;
    }
  }

  private checkAndResetRequestCount(): void {
    const now = Date.now();
    if (now - this.lastResetTime >= 30 * 1000) { // interval 30s 
      this.requestCount = 0;
      this.lastResetTime = now;
    }
  }

  async parseTrendingReposIntoDB(response: any) {
    this.logger.debug(`Github graphql response data length is ${response.data.search.edges.length}`)
    for (const edge of response.data.search.edges) {
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