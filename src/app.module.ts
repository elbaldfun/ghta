import { Module } from '@nestjs/common';
import { ConfigModule } from '@nestjs/config';
import { MongooseModule } from '@nestjs/mongoose';
import configuration from './config/configuration';
import { GithubTrendModule } from './modules/github-trend/github-trend.module';
import { UserModule } from './modules/user/user.module';

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
  ],
  controllers: [],
  providers: [],
})
export class AppModule {}
