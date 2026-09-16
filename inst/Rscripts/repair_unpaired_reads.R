#conda run -n base Rscript R/repair_unpaired_reads.R --bamfile {params.bam_nsorted} --readname {params.bam_readnames}
library(optparse)
option_list = list(make_option(c("--readname"), type = "character", default = NULL, help = "CpG Island BED file, provided by default."))
args = commandArgs(trailingOnly=F)
args <- parse_args(OptionParser(option_list = option_list))
# 获取命令行参数
bam_readnames <- args$readname
readnames <- readLines(bam_readnames)
readnames <- table(readnames)
readnames <- readnames[readnames == 1]
readnames <- names(readnames)
readnames <- readnames[readnames != ""]
if (length(readnames) == 0){
  fs::file_delete(bam_readnames)
} else {
  writeLines(readnames,con = bam_readnames)
}

