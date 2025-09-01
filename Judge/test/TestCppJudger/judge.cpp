#include <assert.h>
#include <fstream>
#include <iostream>

using namespace std;

int main(int argc, const char *argv[]) {
  assert(argc == 2);
  int id = atoi(argv[1]);
  std::ifstream ifs(std::to_string(id) + ".in");
  int n;
  ifs >> n;
  for (int i = 0; i < n; ++i) {
    int x;
    if (!(cin >> x) || x != i) {
      cout << 0 << endl;
      cout << "Fall in line:" << std::to_string(i) <<  " expect:" << std::to_string(i) << endl;
      return 0;
    }
  }

  cout << -1 << endl;
  cout << "OK" << endl;
  return 0;
}