// Числа лабы 0.4 с машины автора: Intel Core i5-12400, Samsung SSD 990 PRO 1 ТБ (NVMe), ext4,
// один полный прогон 2026-10-23 (labs/0-4/README.md, раздел «Машина и результаты»).
// Источник каждого числа — строка «RESULT <ключ> <значение> <единица>» в файле file; тест
// latency.test.ts разбирает её и сверяет с константой, поэтому перепрогон лабы без правки здесь упадёт.
// Всё в наносекундах: так их сравнивает шкала.
export interface LabValue {
  ns: number
  file: string
  key: string
}

const MEM = 'labs/0-4/results/mem-latency.txt'
const SEQ = 'labs/0-4/results/mem-seq-1mb.txt'
const RAND = 'labs/0-4/results/ssd-randread-4k.txt'
const SEQ_SSD = 'labs/0-4/results/ssd-seqread-1m.txt'
const NET = 'labs/0-4/results/net-rtt.txt'

export const LAB_04 = {
  l1: { ns: 1.54, file: MEM, key: 'l1_random_ns' },
  // 256 КиБ, а не весь L2: на 1 МиБ в замер подмешиваются L3 и промахи TLB (README лабы).
  l2: { ns: 4.08, file: MEM, key: 'l2_random_ns' },
  ram: { ns: 100.17, file: MEM, key: 'ram_random_ns' },
  ram1mb: { ns: 59_000, file: SEQ, key: 'ram_seq_1mb_us' },
  // lat (slat + clat) — время, которое видит программа; clat занижает его.
  ssdRandP50: { ns: 63_744, file: RAND, key: 'ssd_rand4k_lat_p50_us' },
  ssdRandP99: { ns: 85_504, file: RAND, key: 'ssd_rand4k_lat_p99_us' },
  ssdSeqP50: { ns: 259_072, file: SEQ_SSD, key: 'ssd_seq1m_lat_p50_us' },
  ssdSeqP99: { ns: 790_528, file: SEQ_SSD, key: 'ssd_seq1m_lat_p99_us' },
  // Плотный ping между двумя контейнерами одной машины — нижняя граница сетевого стека, не ЦОД.
  rttContainer: { ns: 28_000, file: NET, key: 'rtt_container_us' },
} as const satisfies Record<string, LabValue>
