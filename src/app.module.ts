import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { MongooseModule } from '@nestjs/mongoose';
import configuration from './config/configuration';
import { GithubTrendModule } from './modules/github-trend/github-trend.module';
import { UserModule } from './modules/user/user.module';
import { CategoryModule } from './modules/category/category.module';
import { AiService } from './modules/ai/ai.service';

@Module({
  imports: [
    ConfigModule.forRoot({
      isGlobal: true,
      load: [configuration],
    }),
    MongooseModule.forRootAsync({
      useFactory: () => ({
        uri: process.env.MONGODB_URI,
        // connectionFactory: (connection) => {
        //   connection.plugin(require('@meanie/mongoose-to-json'));
        //   return connection;
        // },
      }),
      
    }),
    GithubTrendModule,
    UserModule,
    CategoryModule,
  ],
  controllers: [],
  providers: [AiService],
})
export class AppModule {}
