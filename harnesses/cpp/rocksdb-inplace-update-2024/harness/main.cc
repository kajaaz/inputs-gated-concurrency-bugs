// Harness for the RocksDB in-place update race fixed by commit 933ac0e05
// (PR #12624, first released in v9.3.1). MemTable::Update() rewrites the
// length prefix of a value before it takes the per-key write lock, while a
// concurrent Get() on the same key reads that prefix.
//
// Usage: ./rocksdb-inplace-update <inplace> <sizes>
//
//	<inplace>  1 sets ColumnFamilyOptions::inplace_update_support, 0 does not
//	<sizes>    1 overwrites the key with values of a constant size,
//	           0 makes every overwrite one byte longer than the previous one
//
// The race needs inplace == 1 and sizes == 1: only then is a value updated in
// place, because Update() requires the new value to be no larger than the old.
#include <rocksdb/db.h>
#include <rocksdb/options.h>

#include <atomic>
#include <cstdio>
#include <cstdlib>
#include <string>
#include <thread>
#include <vector>

static const int kOverwrites = 20000;
static const int kReaders = 2;

int main(int argc, char** argv) {
  if (argc != 3 || (argv[1][0] != '0' && argv[1][0] != '1') ||
      (argv[2][0] != '0' && argv[2][0] != '1') || argv[1][1] || argv[2][1]) {
    std::printf("Usage: ./rocksdb-inplace-update <inplace> <sizes>\n");
    return 0;
  }
  const bool inplace = argv[1][0] == '1';
  const bool constant = argv[2][0] == '1';

  char dir[] = "/tmp/rocksdb-inplace-XXXXXX";
  if (mkdtemp(dir) == nullptr) {
    std::perror("mkdtemp");
    return 1;
  }

  rocksdb::Options options;
  options.create_if_missing = true;
  options.inplace_update_support = inplace;
  options.allow_concurrent_memtable_write = !inplace;
  options.write_buffer_size = 256 << 20;

  rocksdb::DB* db = nullptr;
  rocksdb::Status s = rocksdb::DB::Open(options, dir, &db);
  if (!s.ok()) {
    std::printf("open: %s\n", s.ToString().c_str());
    return 1;
  }

  const std::string key = "bomb";
  db->Put(rocksdb::WriteOptions(), key, std::string(64, 'a'));

  std::atomic<bool> done{false};
  std::vector<std::thread> readers;
  for (int r = 0; r < kReaders; ++r) {
    readers.emplace_back([&] {
      std::string value;
      while (!done.load(std::memory_order_relaxed)) {
        db->Get(rocksdb::ReadOptions(), key, &value);
      }
    });
  }

  for (int i = 0; i < kOverwrites; ++i) {
    const size_t size = constant ? 64 : 65 + i;
    db->Put(rocksdb::WriteOptions(), key, std::string(size, 'b' + i % 20));
  }
  done = true;
  for (auto& t : readers) t.join();

  delete db;
  rocksdb::DestroyDB(dir, options);
  std::printf("OK\n");
  return 0;
}
