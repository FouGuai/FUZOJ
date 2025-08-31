#include <iostream>
#include <vector>

int main() {
  std::vector<long long> rls;
  std::ios::sync_with_stdio(false);
  
  for (int i = 1; i <= 1e6; ++i) {
    rls.push_back(i);
  }
  long long sum = 0;
  for (int x : rls) {
    std::cout << sum << '\n';
    sum += x;
  }
  std::cout << sum << std::endl;
  std::cout << "hello world" << std::endl;
  return 0;
}