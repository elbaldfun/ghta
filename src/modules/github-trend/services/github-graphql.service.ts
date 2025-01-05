import { Injectable, Logger } from '@nestjs/common';
import { ConfigService } from '@nestjs/config';
import {inspect} from 'util'
import axios from 'axios';
import { time } from 'console';

@Injectable()
export class GithubGraphqlService {
  private readonly logger = new Logger(GithubGraphqlService.name);
  private readonly apiUrl = 'https://api.github.com/graphql';
  private requestCount = 0;
  private lastResetTime = Date.now();
  private readonly MAX_REQUESTS_PER_HOUR = 5000; // GitHub API 限制

  constructor(private configService: ConfigService) {}

  async fetchAllTrendingRepos(range: string): Promise<any[]> {
    let allTrendingRepos = [];
    let hasNextPage = true;
    let afterCursor: string = ""; // 初始化为 ""
  
    while (hasNextPage) {
      const response = await this.queryTrendingRepos(range, 98, afterCursor);
      if (response.data.search.pageInfo.startCursor === null) {
        this.logger.debug(`Github response no data for range ${range}, because of response.data.search.pageInfo.startCursor is null`)
        break;
      }
      // 处理返回的数据
      const trendingRepos = response.data.search.edges.map(edge => edge.node);
      allTrendingRepos = [...allTrendingRepos, ...trendingRepos];
  
      // 更新分页信息
      hasNextPage = response.data.search.pageInfo.hasNextPage;
      afterCursor = response.data.search.pageInfo.endCursor; // 获取下一个游标
      await new Promise(resolve => setTimeout(resolve, 100));
    }
  
    return allTrendingRepos;
  }


  async queryTrendingRepos(range: string, first: number, after: string = ""): Promise<any> {
    try {
      // 检查并重置计数器
      this.checkAndResetRequestCount();
      
      // 检查是否超出限制
      if (this.requestCount >= this.MAX_REQUESTS_PER_HOUR) {
        throw new Error('Exceeded GitHub API hourly limit');
      }
      // after === "" ? "" : after || "";
      let search = "";
      if (after === "") {
        search = `query: "stars:${range}", type: REPOSITORY, first: ${first}`
      } else {
        search = `query: "stars:${range}", type: REPOSITORY, first: ${first}, after: ${after}`
      }

      this.logger.debug(`search parameters is ${search}`)
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
      
      // this.logger.debug(`graphql response data: ${inspect(response, {showHidden: true, depth: null})}`)
      this.logger.debug(`search query is ${query}`)
      this.logger.debug(`Success to fetch GitHub data: ${response.data.data.search.edges[0]?.node?.url}`);
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
} 