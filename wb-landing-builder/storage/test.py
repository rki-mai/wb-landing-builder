import asyncio
import aiohttp
import json
import time
import random
import statistics
from datetime import datetime
from typing import Dict, List, Any, Optional
import argparse
from dataclasses import dataclass, field
from collections import defaultdict

@dataclass
class TestResult:
    """Результат выполнения одного запроса"""
    operation: str
    status_code: int
    response_time: float
    success: bool
    error_message: str = ""

@dataclass
class TestStatistics:
    """Статистика тестирования"""
    total_requests: int = 0
    successful_requests: int = 0
    failed_requests: int = 0
    response_times: List[float] = field(default_factory=list)
    errors: Dict[str, int] = field(default_factory=dict)
    operations_stats: Dict[str, Dict] = field(default_factory=dict)
    
    def add_result(self, result: TestResult):
        """Добавить результат запроса"""
        self.total_requests += 1
        if result.success:
            self.successful_requests += 1
        else:
            self.failed_requests += 1
            self.errors[result.error_message] = self.errors.get(result.error_message, 0) + 1
        
        self.response_times.append(result.response_time)
        
        # Статистика по операциям
        if result.operation not in self.operations_stats:
            self.operations_stats[result.operation] = {
                "count": 0,
                "success": 0,
                "failed": 0,
                "response_times": []
            }
        
        op_stats = self.operations_stats[result.operation]
        op_stats["count"] += 1
        if result.success:
            op_stats["success"] += 1
        else:
            op_stats["failed"] += 1
        op_stats["response_times"].append(result.response_time)
    
    def print_report(self):
        """Вывести отчет о тестировании"""
        print("\n" + "="*60)
        print("ОТЧЕТ О НАГРУЗОЧНОМ ТЕСТИРОВАНИИ")
        print("="*60)
        
        print(f"\n📊 Общая статистика:")
        print(f"  Всего запросов: {self.total_requests}")
        print(f"  Успешных: {self.successful_requests} ({self.successful_requests/self.total_requests*100:.1f}%)")
        print(f"  Неудачных: {self.failed_requests} ({self.failed_requests/self.total_requests*100:.1f}%)")
        
        if self.response_times:
            print(f"\n⏱️  Время ответа (сек):")
            print(f"  Среднее: {statistics.mean(self.response_times):.3f}")
            print(f"  Медиана: {statistics.median(self.response_times):.3f}")
            if len(self.response_times) >= 20:
                print(f"  95-й перцентиль: {statistics.quantiles(self.response_times, n=20)[18]:.3f}")
            if len(self.response_times) >= 100:
                print(f"  99-й перцентиль: {statistics.quantiles(self.response_times, n=100)[98]:.3f}")
            print(f"  Минимум: {min(self.response_times):.3f}")
            print(f"  Максимум: {max(self.response_times):.3f}")
        
        print(f"\n📈 Статистика по операциям:")
        for op_name, stats in self.operations_stats.items():
            success_rate = stats["success"]/stats["count"]*100 if stats["count"] > 0 else 0
            avg_time = statistics.mean(stats["response_times"]) if stats["response_times"] else 0
            print(f"\n  {op_name.upper()}:")
            print(f"    Всего: {stats['count']}, Успешно: {stats['success']}, Ошибок: {stats['failed']}")
            print(f"    Успешность: {success_rate:.1f}%, Среднее время: {avg_time:.3f}с")
        
        if self.errors:
            print(f"\n❌ Ошибки:")
            for error, count in self.errors.items():
                print(f"  {error}: {count} раз")
        
        print("\n" + "="*60)

class LoadTester:
    """Класс для нагрузочного тестирования API"""
    
    def __init__(self, base_url: str, endpoint: str = "/api/v1/storage"):
        self.base_url = base_url.rstrip('/')
        self.endpoint = endpoint
        self.generated_ids = set()
        self.current_id_counter = 1
        
    def generate_id(self) -> str:
        """Генерация уникального ID для тестовых элементов"""
        element_id = f"lb-{self.current_id_counter}"
        self.current_id_counter += 1
        self.generated_ids.add(element_id)
        return element_id
    
    def generate_create_payload(self) -> Dict:
        """Генерация payload для создания элемента"""
        element_types = ["text", "container", "link", "image", "button"]
        element_type = random.choice(element_types)
        
        payload = {
            "operation": "create",
            "data": {
                "element": element_type,
                "parentId": "root",
                "index": random.randint(0, 10)
            }
        }
        
        # Генерация специфичных полей в зависимости от типа
        if element_type == "text":
            payload["data"]["value"] = f"Тестовый текст {random.randint(1, 1000)}"
            payload["data"]["styles"] = {
                "color": random.choice(["#000000", "#333333", "#666666"]),
                "fontSize": f"{random.choice([12, 14, 16, 18, 24])}px"
            }
        
        elif element_type == "container":
            payload["data"]["styles"] = {
                "display": random.choice(["flex", "block", "grid"]),
                "padding": f"{random.randint(10, 30)}px"
            }
        
        elif element_type == "link":
            payload["data"]["value"] = f"Ссылка {random.randint(1, 100)}"
            payload["data"]["src"] = f"https://example.com/test-{random.randint(1, 1000)}"
            payload["data"]["styles"] = {
                "textDecoration": random.choice(["underline", "none"])
            }
        
        elif element_type == "image":
            payload["data"]["value"] = f"https://cdn.example.com/image-{random.randint(1, 100)}.jpg"
            payload["data"]["alt"] = f"Тестовое изображение {random.randint(1, 100)}"
            payload["data"]["styles"] = {
                "width": "100%",
                "height": "auto"
            }
        
        elif element_type == "button":
            payload["data"]["value"] = f"Кнопка {random.randint(1, 100)}"
            payload["data"]["src"] = f"https://example.com/action-{random.randint(1, 100)}"
            payload["data"]["styles"] = {
                "backgroundColor": random.choice(["#007bff", "#28a745", "#dc3545"]),
                "color": "#ffffff"
            }
        
        return payload
    
    def generate_update_payload(self, element_id: str) -> Dict:
        """Генерация payload для обновления элемента"""
        update_fields = {
            "value": f"Обновленный текст {random.randint(1, 1000)}",
            "styles": {
                "color": random.choice(["#ff0000", "#00ff00", "#0000ff"]),
                "fontSize": f"{random.choice([14, 16, 18, 20])}px"
            }
        }
        
        # Иногда обновляем parentId для имитации перемещения
        if random.random() < 0.3:  # 30% запросов на перемещение
            update_fields["parentId"] = "root"
            update_fields["index"] = random.randint(0, 5)
        
        return {
            "operation": "update",
            "data": {
                "id": element_id,
                "fields": update_fields
            }
        }
    
    def generate_delete_payload(self, element_id: str) -> Dict:
        """Генерация payload для удаления элемента"""
        return {
            "operation": "delete",
            "data": {
                "id": element_id
            }
        }
    
    async def send_request(self, session: aiohttp.ClientSession, payload: Dict) -> TestResult:
        """Отправить один запрос к API"""
        operation = payload.get("operation", "unknown")
        start_time = time.time()
        
        try:
            async with session.post(f"{self.base_url}{self.endpoint}", 
                                   json=payload,
                                   timeout=aiohttp.ClientTimeout(total=10)) as response:
                response_time = time.time() - start_time
                status_code = response.status
                success = 200 <= status_code < 300
                
                if not success:
                    error_text = await response.text()
                    error_message = f"HTTP {status_code}: {error_text[:100]}"
                else:
                    error_message = ""
                
                return TestResult(
                    operation=operation,
                    status_code=status_code,
                    response_time=response_time,
                    success=success,
                    error_message=error_message
                )
                
        except asyncio.TimeoutError:
            response_time = time.time() - start_time
            return TestResult(
                operation=operation,
                status_code=0,
                response_time=response_time,
                success=False,
                error_message="Timeout"
            )
        except Exception as e:
            response_time = time.time() - start_time
            return TestResult(
                operation=operation,
                status_code=0,
                response_time=response_time,
                success=False,
                error_message=str(e)
            )
    
    async def run_load_test(self, 
                           concurrent_users: int = 10,
                           total_requests: int = 100,
                           create_probability: float = 0.4,
                           update_probability: float = 0.4,
                           delete_probability: float = 0.2,
                           step_delay: float = 0.0):
        """
        Запустить нагрузочное тестирование
        
        Args:
            step_delay: Задержка между запусками запросов (в секундах)
                       0.0 - лавинообразная загрузка (все запросы сразу)
                       >0.0 - поэтапный запуск с указанной задержкой
        """
        
        print(f"\n🚀 Запуск нагрузочного тестирования")
        print(f"  URL: {self.base_url}{self.endpoint}")
        print(f"  Concurrent users: {concurrent_users}")
        print(f"  Total requests: {total_requests}")
        print(f"  Create probability: {create_probability}")
        print(f"  Update probability: {update_probability}")
        print(f"  Delete probability: {delete_probability}")
        print(f"  Step delay: {step_delay} сек {'(лавинообразно)' if step_delay == 0 else '(с шагом)'}")
        
        statistics = TestStatistics()
        
        # Создаем очередь запросов
        async with aiohttp.ClientSession() as session:
            semaphore = asyncio.Semaphore(concurrent_users)
            created_ids = []
            
            async def worker(request_data, request_id: int):
                async with semaphore:
                    if step_delay > 0 and request_id > 0:
                        # Для поэтапного запуска - задержка перед выполнением
                        await asyncio.sleep(request_id * step_delay)
                    return await self.send_request(session, request_data)
            
            # Генерируем запросы
            requests_list = []
            for i in range(total_requests):
                # Выбираем тип операции
                rand = random.random()
                
                if rand < create_probability:
                    # CREATE
                    payload = self.generate_create_payload()
                    # Сохраняем ID для будущих операций
                    if "id" not in payload["data"]:
                        new_id = self.generate_id()
                        payload["data"]["id"] = new_id
                        created_ids.append(new_id)
                
                elif rand < create_probability + update_probability and created_ids:
                    # UPDATE
                    element_id = random.choice(created_ids)
                    payload = self.generate_update_payload(element_id)
                
                elif created_ids:
                    # DELETE
                    element_id = random.choice(created_ids)
                    payload = self.generate_delete_payload(element_id)
                    created_ids.remove(element_id)
                
                else:
                    # Если нет элементов для обновления/удаления, создаем новый
                    payload = self.generate_create_payload()
                    new_id = self.generate_id()
                    payload["data"]["id"] = new_id
                    created_ids.append(new_id)
                
                requests_list.append(payload)
            
            # Выполняем запросы с учетом режима запуска
            start_time = time.time()
            
            if step_delay == 0:
                # Лавинообразный режим - все запросы сразу
                tasks = [worker(req, i) for i, req in enumerate(requests_list)]
                results = await asyncio.gather(*tasks)
            else:
                # Поэтапный режим - запускаем с задержкой
                results = []
                for i, req in enumerate(requests_list):
                    result = await worker(req, i)
                    results.append(result)
            
            total_time = time.time() - start_time
            
            # Собираем статистику
            for result in results:
                statistics.add_result(result)
            
            # Выводим отчет
            statistics.print_report()
            
            print(f"\n⏱️  Общее время выполнения: {total_time:.2f} сек")
            print(f"📊 RPS (запросов в секунду): {total_requests/total_time:.2f}")
            
            # Дополнительная информация о режиме запуска
            if step_delay > 0:
                theoretical_time = step_delay * (total_requests - 1) + max(rt.response_time for rt in results)
                print(f"📈 Теоретическое минимальное время: {theoretical_time:.2f} сек")
        
        return statistics

def main():
    parser = argparse.ArgumentParser(description="Нагрузочное тестирование API элементов")
    parser.add_argument("--url", default="http://localhost:8080", help="Базовый URL API")
    parser.add_argument("--endpoint", default="/api/v1/storage/testID/mutations", help="Endpoint API")
    parser.add_argument("--users", type=int, default=10, help="Количество одновременных пользователей")
    parser.add_argument("--requests", type=int, default=5000, help="Общее количество запросов")
    parser.add_argument("--create-ratio", type=float, default=0.4, help="Вероятность создания элемента")
    parser.add_argument("--update-ratio", type=float, default=0.4, help="Вероятность обновления элемента")
    parser.add_argument("--delete-ratio", type=float, default=0.2, help="Вероятность удаления элемента")
    parser.add_argument("--step-delay", type=float, default=0, 
                       help="Задержка между запусками запросов в секундах (0 - лавинообразно, >0 - с шагом)")
    
    args = parser.parse_args()
    
    # Проверка суммы вероятностей
    total_ratio = args.create_ratio + args.update_ratio + args.delete_ratio
    if abs(total_ratio - 1.0) > 0.01:
        print(f"Предупреждение: Сумма вероятностей = {total_ratio}, нормализуем до 1.0")
        args.create_ratio /= total_ratio
        args.update_ratio /= total_ratio
        args.delete_ratio /= total_ratio
    
    # Запуск тестирования
    tester = LoadTester(args.url, args.endpoint)
    
    async def run():
        await tester.run_load_test(
            concurrent_users=args.users,
            total_requests=args.requests,
            create_probability=args.create_ratio,
            update_probability=args.update_ratio,
            delete_probability=args.delete_ratio,
            step_delay=args.step_delay
        )
    
    asyncio.run(run())

if __name__ == "__main__":
    main()