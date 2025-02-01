import { NestFactory } from '@nestjs/core';
import { SwaggerModule, DocumentBuilder } from '@nestjs/swagger';
import { ValidationPipe, Logger } from '@nestjs/common';
import { WinstonModule } from 'nest-winston';
import { AppModule } from './app.module';
import { winstonConfig } from './config/logger/winston.config';
import { mkdir } from 'fs/promises';
import { join } from 'path';

async function bootstrap() {
  // 确保日志目录存在
  const logDir = join(process.cwd(), 'logs');
  await mkdir(logDir, { recursive: true });

  const app = await NestFactory.create(AppModule, {
    logger: WinstonModule.createLogger(winstonConfig),
  });

  // 全局验证管道
  app.useGlobalPipes(new ValidationPipe({
    transform: true,
    whitelist: true,
    forbidNonWhitelisted: true,
  }));

  // Swagger 配置
  const config = new DocumentBuilder()
    .setTitle('GitHub Trend Insights')
    .setDescription('GitHub Trending Insights')
    .setVersion('1.0')
    // .addTag('github-trend')
    .build();
  const document = SwaggerModule.createDocument(app, config);
  SwaggerModule.setup('api', app, document);

  // CORS 配置
  app.enableCors();

  const port = process.env.PORT || 3000;
  await app.listen(port);
  
  const logger = new Logger('Bootstrap');
  logger.log(process.env)
  logger.log(`Application is running on: http://localhost:${port}`);
}
bootstrap();
