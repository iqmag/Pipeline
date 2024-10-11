package main

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
    bufferDrainInterval = 30 * time.Second // Интервал очистки кольцевого буфера
    bufferSize = 10 // Размер кольцевого буфера
)

// Структура RingIntBuffer - кольцевой буфер целых чисел
type RingIntBuffer struct {
    array []int // Массив для хранения элементов буфера
    pos   int // Текущая позиция для записи в буфер
    size  int // Размер буфера
    mu     sync.Mutex // Мьютекс для обеспечения потокобезопасности
}

// Функция NewRingIntBuffer - создание нового буфера целых чисел
func NewRingIntBuffer(size int) *RingIntBuffer {
    return &RingIntBuffer {
        array: make([]int, size),
        pos: -1,
        size: size,
    }
}

// Метод Push - добавление нового элемента в буфер
func (r *RingIntBuffer) Push(el int) {
    r.mu.Lock()
    defer r.mu.Unlock()

    r.pos = (r.pos + 1) % r.size // Вычисление новой позиции для записи с учетом размера буфера
    r.array[r.pos] = el // Запись элемента в буфер
}

// Метод Get - получение всех элементов буфера и его последующая очистка
func (r *RingIntBuffer) Get() []int {
    // Проверка, если буфер пуст (позиция меньше 0), возвращаем nil
    if r.pos < 0 {
        return nil
    }
    r.mu.Lock() // Блокировка мьютекса для обеспечения безопасности доступа к буферу
    defer r.mu.Unlock() // Отложенное разблокирование мьютекса после выполнения функции
    var output []int = r.array[:r.pos+1] // Создание слайса, который включает все элементы от начала буфера r.array до индекса r.pos включительно
    r.pos = -1 // Виртуальная очистка нашего буфера, устанавливаем позицию в -1

    return output // Возвращаем слайс с элементами буфера
}

// StageInt - Стадия конвейера, обрабатывающая целые числа
type StageInt func(<-chan int, <-chan struct{}) <-chan int

// Структура PipelineInt - Пайплайн обработки целых чисел
type PipelineInt struct {
    stages []StageInt // Список стадий конвейера
    done   <-chan struct{} // Канал завершения работы
}

// Функция NewPipelineInt - Создание пайплайна обработки целых чисел
func NewPipelineInt(done <-chan struct{}, stages ...StageInt) *PipelineInt {
    return &PipelineInt {
        done: done,
        stages: stages,
    }
}

// Метод Run - Запуск пайплайна обработки целых чисел
func (p *PipelineInt) Run(source <-chan int) <-chan int {
    c := source // Инициализация канала для передачи данных
    for _, stage := range p.stages {
        c = stage(c, p.done) // Передача данных через каждую стадию
    }

    return c
}

func main() {
    source, done := dataSource() // Источник данных и канал завершения работы

    // Создаем пайплайн и передаем ему стадии
    pipeline := NewPipelineInt(done,
        negativeFilterStageInt, // Фильтрация отрицательных чисел
        specialFilterStageInt, // Фильтрация чисел, кратных 3, исключая 0
        bufferStageInt, // Буферизация данных
    )

    // Потребитель данных от пайплайна
    consumer(done, pipeline.Run(source))
}

// Функция dataSource - Создание источника данных
func dataSource() (<-chan int, <-chan struct{}) {
    c := make(chan int) // Канал для передачи данных
    done := make(chan struct{}) // Канал для завершения работы

    go func() {
        defer close(done)
        scanner := bufio.NewScanner(os.Stdin)
        log.Println("Введите целые числа. Для выхода введите 'exit':")

        for {
            scanner.Scan()
            data := scanner.Text()
            if strings.EqualFold(data, "exit") {
                close(c)
                log.Println("Программа завершила работу!")

                return
            }

            i, err := strconv.Atoi(data)
            if err != nil {
                log.Println("Ошибка ввода, программа обрабатывает только целые числа!")

                continue
            }

            c <- i // Отправка целого числа в канал
        }
    }()

    return c, done
}

// Функция negativeFilterStageInt - Стадия фильтрации отрицательных чисел
func negativeFilterStageInt(input <-chan int, done <-chan struct{}) <-chan int {
    output := make(chan int) // Канал для передачи отфильтрованных данных

    go func() {
        defer close(output)
        for {
            select {
            case data, ok := <-input: // Чтение данных из входного канала
                if !ok {

                    return
                }
                if data >= 0 { // Если число не отрицательное
                    select {
                    case output <- data: // Отправка числа в выходной канал
                    case <-done: // Проверка канала завершения работы

                        return
                    }
                }
            case <-done: // Если пришел сигнал завершения работы

                return
            }
        }
    }()

    return output
}

// Функция specialFilterStageInt - Стадия фильтрации чисел, не кратных 3, исключая также и 0
func specialFilterStageInt(input <-chan int, done <-chan struct{}) <-chan int {
    output := make(chan int) // Канал для передачи отфильтрованных данных

    go func() {
        defer close(output)
        for {
            select {
            case data, ok := <-input: // Чтение данных из входного канала
                if !ok {

                    return
                }
                if data != 0 && data%3 == 0 { // Если число кратно 3 и не равно 0
                    select {
                    case output <- data: // Отправка числа в выходной канал
                    case <-done: // Проверка канала завершения работы
                        
                        return
                    }
                }
            case <-done: // Если пришел сигнал завершения работы

                return
            }
        }
    }()

    return output
}

// Функция bufferStageInt - Стадия буферизации данных
func bufferStageInt(input <-chan int, done <-chan struct{}) <-chan int {
    output := make(chan int) // Канал для передачи буферизованных данных
    buffer := NewRingIntBuffer(bufferSize) // Кольцевой буфер

    var wg sync.WaitGroup

    wg.Add(1)
    go func() {
        defer wg.Done()
        for {
            select {
            case data, ok := <-input: // Чтение данных из входного канала
                if !ok {

                    return
                }
                buffer.Push(data) // Добавление данных в буфер
            case <-done: // Если пришел сигнал завершения работы

                return
            }
        }
    }()
    // В этой стадии есть вспомогательная горутина, выполняющая просмотр буфера с заданным интервалом времени - bufferDrainInterval
    wg.Add(1)
    go func() {
        defer wg.Done()
        for {
            select {
            case <-time.After(bufferDrainInterval): // Через заданный интервал времени
                data := buffer.Get() // Получение данных из буфера
                if data != nil {
                    for _, v := range data {
                        select {
                        case output <- v: // Отправка элемента в выходной канал
                        case <-done: // Проверка канала завершения работы

                            return
                        }
                    }
                }
            case <-done: // Если пришел сигнал завершения работы

                return
            }
        }
    }()

    go func() {
        wg.Wait()
        close(output)
        }()

        return output
}

// Функция consumer - Потребитель данных из канала
func consumer(done <-chan struct{}, c <-chan int) {
    for {
        select {
        case data, ok := <-c: // Чтение данных из канала
            if !ok {

                return
            }
            log.Printf("Обработаны данные: %d\n", data)
        case <-done: // Если пришел сигнал завершения работы

            return
        }
    }
}