library(optparse)
option_list = list(make_option(c("--filein"), type = "character", default = NULL, help = "CpG Island BED file, provided by default."),
                   make_option(c("--fileout"), type = "character", default = "human", help = "species1, provided by default."))
args = commandArgs(trailingOnly=F)
args <- parse_args(OptionParser(option_list = option_list))
# 获取命令行参数
filein <- args$filein
fileout <- args$fileout
message(fileout)
command <- glue::glue("awk '($4 > 0) || ($5 > 0) {{print $1\".\"$2,$1,$2,$3,$4+$5,100*$4/($4+$5),100*$5/($4+$5)}}' {filein} | sed 's/+/F/g' | sed 's/-/R/g' | sed 's/[[:space:]]/\\t/g' > {fileout}")
system(command = command)
