#include "fuzoj_utils.h"
#include <fcntl.h>
#include <ftw.h>
#include <sys/sendfile.h>
#include <sys/stat.h>
#include <unistd.h>
namespace fuzoj {
std::string Utils::GetFileName(const std::string &path) {
  int i = static_cast<int>(path.size()) - 1;
  for (; i >= 0; --i) {
    if (path[i] == '/') {
      break;
    }
  }

  return path.substr(i + 1, static_cast<int>(path.size()) - i - 1);
}

int Utils::CopyFile(const std::string &dst, const std::string &src) {
  int source = open(src.c_str(), O_RDONLY);
  if (source < 0) return -1;

  struct stat stat_buf;
  fstat(source, &stat_buf);

  int dest = open(dst.c_str(), O_WRONLY | O_CREAT | O_TRUNC, stat_buf.st_mode);
  if (dest < 0) {
    close(source);
    return -1;
  }

  off_t offset = 0;
  if (sendfile(dest, source, &offset, stat_buf.st_size) < 0) {
    close(source);
    close(dest);
    return -1;
  }

  close(source);
  close(dest);
  return 0;
}

static int Remove(const char *fpath, const struct stat *sb, int typeflag, struct FTW *ftwbuf) {
  (void)sb;
  (void)ftwbuf;
  int ret = remove(fpath);
  if (ret) {
    perror(fpath);
  }
  return ret;
}

int Utils::RemoveDirRecursive(const std::string &path) { return nftw(path.c_str(), Remove, 64, FTW_DEPTH | FTW_PHYS); }
}  // namespace fuzoj