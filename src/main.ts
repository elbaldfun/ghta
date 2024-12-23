import { NestFactory } from '@nestjs/core';
import { SwaggerModule, DocumentBuilder } from '@nestjs/swagger';
import { ValidationPipe } from '@nestjs/common';
import { AppModule } from './app.module';

async function bootstrap() {
  const app = await NestFactory.create(AppModule);

  // 全局验证管道
  app.useGlobalPipes(new ValidationPipe({
    transform: true,
    whitelist: true,
    forbidNonWhitelisted: true,
  }));

  // Swagger 配置
  const config = new DocumentBuilder()
    .setTitle('GitHub Trend API')
    .setDescription('GitHub Trending 仓库数据采集和查询服务')
    .setVersion('1.0')
    .addTag('github-trend')
    .build();
  const document = SwaggerModule.createDocument(app, config);
  SwaggerModule.setup('api', app, document);

  // CORS 配置
  app.enableCors();

  const port = process.env.PORT || 3000;
  await app.listen(port);
  console.log(process.env);
  console.log(`Application is running on: http://localhost:${port}`);
}
bootstrap();
